#include <Python.h>


#define PY_BUILD_VALUE_BYTES "y#"
#define PyString_FromStringAndSize PyBytes_FromStringAndSize
#define PyString_AsStringAndSize PyBytes_AsStringAndSize
#define PyString_Size PyBytes_Size
#define PyInt_CheckExact(obj) 0


struct benc_state {
    unsigned int cast : 1;

    int size;
    int offset;
    char* buffer;
    PyObject* file;

    PyObject** references_stack;
    int references_size;
    int references_top;
};


PyObject* BencodeValueError;
PyObject* BencodeTypeError;


static void benc_state_init(struct benc_state* bs) {
    bs->size = 256;
    bs->offset = 0;
    bs->buffer = malloc(bs->size);
    bs->file = NULL;

    bs->references_size = 8;
    bs->references_top = 0;
    bs->references_stack = malloc(sizeof(PyObject*) * bs->references_size);
}


static void benc_state_free(struct benc_state* bs) {
    if (bs->buffer != NULL) {
        free(bs->buffer);
    }
    if (bs->references_stack != NULL) {
        free(bs->references_stack);
    }
}


static void benc_state_flush(struct benc_state* bs) {
    if (bs->offset > 0) {
        PyObject_CallMethod(bs->file, "write", PY_BUILD_VALUE_BYTES, bs->buffer, bs->offset);
        bs->offset = 0;
    }
}


static void benc_state_write_char(struct benc_state* bs, char c) {
    if (bs->file == NULL) {
        if ((bs->offset + 1) >= bs->size) {
            bs->buffer = realloc(bs->buffer, bs->size * 2);
        }
        bs->buffer[bs->offset++] = c;
    } else {
        if ((bs->offset + 1) >= bs->size) {
            PyObject_CallMethod(bs->file, "write", PY_BUILD_VALUE_BYTES, bs->buffer, bs->offset);
            bs->offset = 0;
        }
        bs->buffer[bs->offset++] = c;
    }
}


static void benc_state_write_buffer(struct benc_state* bs, char* buff, int size) {
    if (bs->file == NULL) {
        int new_size;
        for (new_size = bs->size; new_size <= (bs->offset + size); new_size *= 2);
        if (new_size > bs->size) {
            bs->buffer = realloc(bs->buffer, new_size);
            bs->size = new_size;
        }
        memcpy(bs->buffer + bs->offset, buff, size);
        bs->offset += size;
    } else {
        if (bs->offset + size >= bs->size) {
            PyObject_CallMethod(bs->file, "write", PY_BUILD_VALUE_BYTES, bs->buffer, bs->offset);
            bs->offset = 0;
        }
        if (size >= bs->size) {
            PyObject_CallMethod(bs->file, "write", PY_BUILD_VALUE_BYTES, buff, size);
        } else {
            memcpy(bs->buffer + bs->offset, buff, size);
            bs->offset += size;
        }
    }
}


static void benc_state_write_format(struct benc_state* bs, const int limit, const void *format, ...) {
    char buffer[limit + 1]; // moze by malloca()?

    va_list ap;
    va_start(ap, format);
    int size = vsnprintf(buffer, limit, format, ap);
    va_end(ap);

    return benc_state_write_buffer(bs, buffer, (size < limit) ? size : (limit - 1));
}


static int benc_state_read_char(struct benc_state* bs) {
    if (bs->file == NULL) {
        if (bs->offset < bs->size) {
            return bs->buffer[bs->offset++];
        } else {
            return -1;
        }
    } else {
        char *buffer;
        int result;
        Py_ssize_t length;
        PyObject *data =  PyObject_CallMethod(bs->file, "read", "i", 1);
        if (-1 == PyString_AsStringAndSize(data, &buffer, &length)) {
            return -1;
        }
        if (length == 1) {
            result = buffer[0];
        } else {
            result = -1;
        }
        Py_DECREF(data);
        return result;
    }
}


static PyObject *benc_state_read_pystring(struct benc_state* bs, int size) {
    if (bs->file == NULL) {
        if (bs->offset + size <= bs->size) {
            PyObject *result = PyString_FromStringAndSize(bs->buffer + bs->offset, size);
            bs->offset += size;
            return result;
        } else {
            PyErr_Format(
                BencodeValueError,
                "unexpected end of data"
            );
            return NULL;
        }
    } else {
        PyObject *result = PyObject_CallMethod(bs->file, "read", "i", size);
        if (PyString_Size(result) == size) {
            return result;
        } else {
            Py_DECREF(result);
            PyErr_Format(
                BencodeValueError,
                "unexpected end of data"
            );
            return NULL;
        }
    }
}

static void benc_state_references_push(struct benc_state* bs, PyObject *obj) {
    if ((bs->references_top + 1) == bs->references_size) {
        bs->references_size *= 2;
        bs->references_stack = realloc(
            bs->references_stack,
            sizeof(PyObject*) * bs->references_size
        );
    }
    bs->references_stack[bs->references_top++] = obj;
}

static void benc_state_references_pop(struct benc_state* bs) {
    bs->references_top--;
}

static int benc_state_references_contains(struct benc_state* bs, PyObject *obj) {
    int i;
    for (i = 0; i < bs->references_top; i++) {
        if (bs->references_stack[i] == obj) {
            return 1;
        }
    }
    return 0;
}


static int do_dump(struct benc_state *bs, PyObject* obj);

static int do_dump(struct benc_state *bs, PyObject* obj) {
    int i = 0, n = 0;

    if (benc_state_references_contains(bs, obj)) {
        PyErr_Format(
            BencodeValueError,
            "circular reference detected"
        );
        return 0;
    }

    if (PyBytes_CheckExact(obj)) {
        char *buff = PyBytes_AS_STRING(obj);
        int size = PyBytes_GET_SIZE(obj);

        benc_state_write_format(bs, 12, "%d:", size);
        benc_state_write_buffer(bs, buff, size);
    } else if (PyInt_CheckExact(obj) || PyLong_CheckExact(obj)) {
        long x = PyLong_AsLong(obj);
        benc_state_write_format(bs, 23, "i%lde", x);
    } else if (bs->cast && PyBool_Check(obj)) {
        long x = PyLong_AsLong(obj);
        benc_state_write_format(bs, 4, "i%lde", x);
    } else if (PyList_CheckExact(obj) || (bs->cast && PyList_Check(obj))) {
        n = PyList_GET_SIZE(obj);
        benc_state_references_push(bs, obj);
        benc_state_write_char(bs, 'l');
        for (i = 0; i < n; i++) {
            do_dump(bs, PyList_GET_ITEM(obj, i));
        }
        benc_state_write_char(bs, 'e');
        benc_state_references_pop(bs);
    } else if (bs->cast && PyTuple_Check(obj)) {
        n = PyTuple_GET_SIZE(obj);
        benc_state_references_push(bs, obj);
        benc_state_write_char(bs, 'l');
        for (i = 0; i < n; i++) {
            do_dump(bs, PyTuple_GET_ITEM(obj, i));
        }
        benc_state_write_char(bs, 'e');
        benc_state_references_pop(bs);
    } else if (PyDict_CheckExact(obj)) {
        Py_ssize_t index = 0;
        PyObject *keys, *key, *value;
        keys = PyDict_Keys(obj);
        PyList_Sort(keys);

        benc_state_references_push(bs, obj);
        benc_state_write_char(bs, 'd');
        for (index = 0; index < PyList_Size(keys); index++) {
            key = PyList_GetItem(keys, index);
            value = PyDict_GetItem(obj, key);
            do_dump(bs, key);
            do_dump(bs, value);
        }
        benc_state_write_char(bs, 'e');
        benc_state_references_pop(bs);

        Py_DECREF(keys);
    } else {
        PyErr_Format(
            BencodeTypeError,
            "type %s is not Bencode serializable",
            Py_TYPE(obj)->tp_name
        );
    }
    return 0;
}


static PyObject* dumps(PyObject* self, PyObject* args, PyObject* kwargs) {
    static char *kwlist[] = {"obj", "cast", NULL};

    PyObject* obj;
    PyObject* result;
    int cast = 0;

    struct benc_state bs;
    benc_state_init(&bs);

    if (!PyArg_ParseTupleAndKeywords(
        args, kwargs, "O|i", kwlist,
        &obj, &cast
    ))
    {
        return NULL;
    }

    bs.cast = !!cast;

    do_dump(&bs, obj);

    if (PyErr_Occurred()) {
        benc_state_free(&bs);
        return NULL;
    } else {
        result = Py_BuildValue(PY_BUILD_VALUE_BYTES, bs.buffer, bs.offset);
        benc_state_free(&bs);
        return result;
    }
}


static PyObject *do_load(struct benc_state *bs) {
    PyObject *retval = NULL;

    int first = benc_state_read_char(bs);

    switch (first) {
        case 'i': {
            int sign = 1;
            int read_cnt = 0;
            long long value = 0;
            int current = benc_state_read_char(bs);
            if (current == '-') {
                sign = -1;
                current = benc_state_read_char(bs);
            }
            while (('0' <= current) && (current <= '9')) {
                value = value * 10 + (current - '0');
                current = benc_state_read_char(bs);
                read_cnt++;
            }

            if ('e' == current) {
                if (read_cnt > 0) {
                    value *= sign;
                    retval = PyLong_FromLongLong(value);
                } else {
                    PyErr_Format(
                        BencodeValueError,
                        "unexpected end of data"
                    );
                    retval = NULL;
                }
            } else if (-1 == current) {
                PyErr_Format(
                    BencodeValueError,
                    "unexpected end of data"
                );
                retval = NULL;
            } else {
                PyErr_Format(
                    BencodeValueError,
                    "unexpected byte 0x%.2x",
                    current
                );
                retval = NULL;
            }

            } break;

        case '0':
        case '1':
        case '2':
        case '3':
        case '4':
        case '5':
        case '6':
        case '7':
        case '8':
        case '9': {
            int size = first - '0';
            char current = benc_state_read_char(bs);
            while (('0' <= current) && (current <= '9')) {
                size = size * 10 + (current - '0');
                current = benc_state_read_char(bs);
            }
            if (':' == current) {
                retval = benc_state_read_pystring(bs, size);
            } else if (-1 == current) {
                PyErr_Format(
                    BencodeValueError,
                    "unexpected end of data"
                );
                retval = NULL;
            } else {
                PyErr_Format(
                    BencodeValueError,
                    "unexpected byte 0x%.2x",
                    current
                );
                retval = NULL;
            }

            } break;
        case 'e':
            Py_INCREF(PyExc_StopIteration);
            retval = PyExc_StopIteration;
            break;
        case 'l': {
            PyObject *v = PyList_New(0);
            PyObject *item;

            while (1) {
                item = do_load(bs);

                if (item == PyExc_StopIteration) {
                    Py_DECREF(PyExc_StopIteration);
                    break;
                }

                if (item == NULL) {
                    if (!PyErr_Occurred()) {
                        PyErr_SetString(
                            BencodeTypeError,
                            "unexpected error in list"
                        );
                    }
                    Py_DECREF(v);
                    v = NULL;
                    break;
                }

                PyList_Append(v, item);
                Py_DECREF(item);
            }

            retval = v;
            } break;
        case 'd': {
            PyObject *v = PyDict_New();

            while (1) {
                PyObject *key, *val;
                key = val = NULL;
                key = do_load(bs);
                
                if (key == PyExc_StopIteration) {
                    Py_DECREF(PyExc_StopIteration);
                    break;
                }

                if (key == NULL) {
                    if (!PyErr_Occurred()) {
                        PyErr_SetString(BencodeTypeError, "unexpected error in dict");
                    }
                    break;
                }

                val = do_load(bs);
                if (val != NULL) {
                    PyDict_SetItem(v, key, val);
                } else {
                    if (!PyErr_Occurred()) {
                        PyErr_SetString(BencodeTypeError, "unexpected error in dict");
                    }
                    break;
                }
                Py_DECREF(key);
                Py_XDECREF(val);
            }
            if (PyErr_Occurred()) {
                Py_DECREF(v);
                v = NULL;
            }
            retval = v;
            } break;
        case -1: {
            PyErr_Format(
                BencodeValueError,
                "unexpected end of data"
            );
            retval = NULL;
            } break;
        default:
            PyErr_Format(
                BencodeValueError,
                "unexpected byte 0x%.2x",
                first
            );
            retval = NULL;
            break;
    }
    return retval;
}


static PyObject* loads(PyObject* self, PyObject* args) {
    struct benc_state bs;
    memset(&bs, 0, sizeof(struct benc_state));

    if (!PyArg_ParseTuple(args, PY_BUILD_VALUE_BYTES, &(bs.buffer), &(bs.size)))
        return NULL;

    PyObject* obj = do_load(&bs);

    return obj;
}


static PyObject* loads2(PyObject* self, PyObject* args) {
    /* TODO:
     *
     * PyLong_FromLong and PyTuple_Pack might return NULL. How to handle these errors?
     */
    struct benc_state bs;
    memset(&bs, 0, sizeof(struct benc_state));

    if (!PyArg_ParseTuple(args, PY_BUILD_VALUE_BYTES, &(bs.buffer), &(bs.size)))
        return NULL;

    PyObject* obj = do_load(&bs);
    PyObject* offset =  PyLong_FromLong((long) bs.offset);

    return PyTuple_Pack(2, obj, offset);
}


static PyObject *add_errors(PyObject *module) {
    BencodeValueError = PyErr_NewException(
        "bencoder._fast.BencodeValueError", PyExc_ValueError, NULL
    );
    Py_INCREF(BencodeValueError);
    PyModule_AddObject(module, "BencodeValueError", BencodeValueError);

    BencodeTypeError = PyErr_NewException(
        "bencoder._fast.BencodeTypeError", PyExc_TypeError, NULL
    );
    Py_INCREF(BencodeTypeError);
    PyModule_AddObject(module, "BencodeTypeError", BencodeTypeError);

    return module;
}


static PyMethodDef bencoder_fastMethods[] = {
    {"loads", loads, METH_VARARGS, "Deserialize ``s`` to a Python object."},
    {"loads2", loads2, METH_VARARGS, "Deserialize ``s`` to a Python object and return end index."},
    {"dumps", dumps, METH_VARARGS|METH_KEYWORDS, "Serialize ``obj`` to a Bencode formatted ``str``."},
    {NULL, NULL, 0, NULL}
};


static struct PyModuleDef bencoder_fast_module = {
    PyModuleDef_HEAD_INIT,
    "bencoder._fast",
    NULL,
    -1,
    bencoder_fastMethods,
    NULL,
    NULL,
    NULL,
    NULL
};

PyMODINIT_FUNC
PyInit__fast(void) {
    PyObject *module = PyModule_Create(&bencoder_fast_module);
    return add_errors(module);

}
